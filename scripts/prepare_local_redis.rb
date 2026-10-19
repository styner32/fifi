#!/usr/bin/env ruby
# Prepare this Compose project's local Redis volume without deleting either copy.
require 'fileutils'
require 'json'
require 'open3'
require 'shellwords'

class LocalRedisPreparation
  PROBE = <<~'SH'.freeze
    if [ -f /target/.fifi-redis8-ready ]; then
      mode="$(cat /target/.fifi-redis8-appendonly 2>/dev/null || true)"
      case "$mode" in yes|no) printf 'ready=%s\n' "$mode"; exit 0 ;; esac
      printf 'invalid_ready=yes\n'; exit 0
    fi
    [ -z "$(find /target -mindepth 1 -maxdepth 1 -print -quit)" ] || { printf 'nonempty_target=yes\n'; exit 0; }
    printf 'source_identity=%s\n' "$(stat -c '%d:%i' /source)"
    [ -n "$(find /source -mindepth 1 -maxdepth 1 -print -quit)" ] || printf 'fresh=yes\n'
    [ -z "$(find /source -type l -print -quit)" ] || printf 'symlinks=yes\n'
    [ ! -f /source/dump.rdb ] || printf 'rdb=yes\n'
    [ ! -f /source/appendonly.aof ] || printf 'aof_legacy=yes\n'
    [ ! -s /source/appendonlydir/appendonly.aof.manifest ] || printf 'aof_manifest=yes\n'
  SH

  def initialize
    @compose = Shellwords.split(ENV.fetch('DOCKER_COMPOSE', 'docker compose'))
    raise 'DOCKER_COMPOSE must contain a command.' if @compose.empty?
  end

  def command(*arguments)
    # Print command names, never config output, Redis values, or environment data.
    warn "[redis-prepare] compose #{arguments.first(2).join(' ')}"
    output, _diagnostic, status = Open3.capture3(*@compose, *arguments)
    raise "Compose #{arguments.first} failed (exit #{status.exitstatus}); no data was deleted. Check the local Docker/Compose service." unless status.success?
    output
  end

  def probe
    command('--profile', 'local-init', 'run', '--rm', '--no-deps', '-T', '--entrypoint', '/bin/sh', 'redis-data-clone', '-ec', PROBE).lines.to_h do |line|
      key, value = line.strip.split('=', 2)
      [key, value]
    end
  end

  def prepare
    state = probe
    if state['ready']
      warn '[redis-prepare] Redis target is already prepared; services were left running.'
      return
    end
    raise 'Redis target has an invalid readiness marker. Both volumes are preserved; inspect the target before retrying.' if state['invalid_ready']
    raise 'Redis target is nonempty and unprepared. Refusing to replace it; both volumes are preserved.' if state['nonempty_target']
    raise 'The original Redis volume contains symlinks. Automatic local initialization cannot safely copy this layout.' if state['symlinks']

    config = JSON.parse(command('config', '--format', 'json'))
    services = config.fetch('services')
    records = command('ps', '-a', '--format', 'json', 'redis').strip
    containers = if records.empty?
                   []
                 elsif records.start_with?('[')
                   JSON.parse(records)
                 else
                   records.lines.map { |line| JSON.parse(line) }
                 end
    raise 'Multiple local Redis containers found; automatic initialization requires one Redis service.' if containers.length > 1
    current = containers.first
    running = current && current['State'] == 'running'
    if current && !running && !%w[exited created dead].include?(current['State'])
      raise 'Local Redis is not running or cleanly stopped. Resolve its paused/restarting state before retrying.'
    end

    if running
      values = command('exec', '-T', 'redis', 'redis-cli', '--raw', 'CONFIG', 'GET', 'appendonly', 'dir', 'dbfilename', 'appenddirname', 'appendfilename').lines.map(&:strip)
      raise 'Cannot read Redis persistence configuration. Unlock/authenticate the local Redis service before retrying.' unless values.length.even?
      settings = values.each_slice(2).to_h
      mode = settings['appendonly']
      raise 'Redis appendonly setting must be yes or no.' unless %w[yes no].include?(mode)
      valid = settings['dir'] == '/data' && settings['dbfilename'] == 'dump.rdb'
      valid &&= settings['appendfilename'] == 'appendonly.aof' && [nil, 'appendonlydir'].include?(settings['appenddirname']) if mode == 'yes'
      raise 'Local Redis uses custom persistence paths. Expected /data/dump.rdb and default AOF filenames; both volumes are unchanged.' unless valid
      identity = command('exec', '-T', 'redis', 'sh', '-c', "stat -c '%d:%i' /data").strip
      raise 'The running Redis /data mount is not the original redis-data volume. Refusing to stop or copy a different data source.' unless identity == state['source_identity']
      writers = %w[dart-filing-api dart-filing-worker].select { |name| services.key?(name) }
      command('stop', '--timeout', '30', *writers) unless writers.empty?
      saved = command('exec', '-T', 'redis', 'redis-cli', '--raw', 'SAVE').strip
      raise 'Redis SAVE did not confirm OK. The original volume is preserved; Redis was not stopped or cloned.' unless saved == 'OK'
      command('stop', '--timeout', '30', 'redis')
      state = probe
    else
      mode = stopped_mode(state)
    end
    raise 'Source changed or the target became nonempty during initialization. Both copies are preserved.' if state['nonempty_target'] || state['ready'] || state['invalid_ready'] || state['symlinks']
    if mode == 'yes' && !state['aof_legacy'] && !state['aof_manifest']
      raise 'AOF mode was selected, but no default AOF file/manifest exists in the stopped source. Nothing was copied.'
    end
    if mode == 'no' && !state['fresh'] && !state['rdb']
      raise 'RDB mode was selected, but /data/dump.rdb is missing from the stopped source. Nothing was copied.'
    end
    if mode == 'yes' && state['aof_legacy'] && state['aof_manifest']
      raise 'Both legacy and manifest AOF layouts exist. Their authority is ambiguous; preserve both volumes and inspect the source before retrying.'
    end
    command('--profile', 'local-init', 'run', '--rm', '--no-deps', '-T', '-e', "REDIS_APPENDONLY=#{mode}", 'redis-data-clone')
    raise 'Redis target readiness could not be verified; inspect the preserved volumes.' unless probe['ready'] == mode
    warn "[redis-prepare] Local Redis target prepared (appendonly=#{mode}); original redis-data is preserved."
  end

  def stopped_mode(state)
    override = ENV['REDIS_APPENDONLY']
    raise 'REDIS_APPENDONLY must be yes or no when supplied.' if override && !%w[yes no].include?(override)
    return 'no' if state['fresh']
    aof = state['aof_legacy'] || state['aof_manifest']
    raise 'No default Redis persistence files were found in the nonempty source. Inspect its custom layout before retrying; both volumes are preserved.' unless aof || state['rdb']
    return override if override
    return 'yes' if aof && !state['rdb']
    return 'no' if state['rdb'] && !aof
    raise 'Stopped Redis has both RDB and AOF evidence, or no recognized persistence files. Set REDIS_APPENDONLY=yes|no only from the last known source configuration; both volumes are preserved.'
  end
end

if $PROGRAM_NAME == __FILE__
  begin
    Dir.chdir(File.expand_path('..', __dir__))
    FileUtils.mkdir_p('.cache')
    File.open('.cache/redis-prepare.lock', File::RDWR | File::CREAT, 0o600) do |lock|
      lock.flock(File::LOCK_EX)
      LocalRedisPreparation.new.prepare
    end
  rescue JSON::ParserError
    warn '[redis-prepare] Cannot parse Docker Compose metadata; configuration output was suppressed to protect local secrets.'
    exit 1
  rescue StandardError => error
    warn "[redis-prepare] #{error.message}"
    exit 1
  end
end
