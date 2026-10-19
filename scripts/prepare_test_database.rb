#!/usr/bin/env ruby
# Creates only a local Compose test database; never drops or resets an existing DB.
require 'json'
require 'open3'
require 'shellwords'
require 'uri'

def capture!(*command, input: '')
  output, errors, status = Open3.capture3(*command, stdin_data: input)
  $stderr.write(errors)
  abort "Local database command failed (#{command.first})." unless status.success?
  output
end

begin
  connection = URI.parse(ENV.fetch('TEST_DATABASE_URL'))
  database = URI::DEFAULT_PARSER.unescape(connection.path.to_s.delete_prefix('/'))
  unless %w[postgres postgresql].include?(connection.scheme) && database.end_with?('_test') && !database.include?("\0")
    abort 'TEST_DATABASE_URL must select a PostgreSQL *_test database.'
  end
  unless %w[localhost 127.0.0.1 ::1].include?(connection.hostname)
    abort 'test-db-prepare creates only a local Compose database. Prepare an external test DB separately, then run make migrate-test-up.'
  end
  compose = Shellwords.split(ENV.fetch('DOCKER_COMPOSE', 'docker compose'))
  abort 'DOCKER_COMPOSE must not be empty.' if compose.empty?
  configuration = JSON.parse(capture!(*compose, 'config', '--format', 'json'))
  service = configuration.fetch('services').fetch('db')
  ports = service.fetch('ports', []).select { |entry| entry.fetch('target').to_i == 5432 }.map { |entry| entry.fetch('published').to_i }
  unless ports.include?(connection.port || 5432)
    abort 'TEST_DATABASE_URL port does not match the local Compose db port. Prepare the selected DB separately, then run make migrate-test-up.'
  end
  administrator = service.fetch('environment').fetch('POSTGRES_USER')
  owner = connection.user ? URI::DEFAULT_PARSER.unescape(connection.user) : administrator
  puts capture!(*compose, 'up', '-d', '--wait', 'db')
  sql = <<~'SQL'
    SELECT format('CREATE DATABASE %I OWNER %I', :'fifi_database', :'fifi_owner')
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'fifi_database')
    \gexec
  SQL
  # psql quotes identifiers through format(%I); no untrusted text is SQL code.
  puts capture!(*compose, 'exec', '-T', 'db', 'psql', '-X', '-v', 'ON_ERROR_STOP=1',
                '-U', administrator, '-d', 'postgres', '-v', "fifi_database=#{database}", '-v', "fifi_owner=#{owner}", input: sql)
  puts 'Local test database is ready; existing contents were preserved.'
rescue KeyError, URI::InvalidURIError, JSON::ParserError, ArgumentError, Errno::ENOENT
  abort 'Cannot prepare the local test database. Check TEST_DATABASE_URL, Docker Compose and the db service configuration.'
end
