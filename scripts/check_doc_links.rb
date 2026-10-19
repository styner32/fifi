#!/usr/bin/env ruby
require 'uri'

root = File.expand_path('..', __dir__)
files = %w[README.md AGENTS.md web/README.md].map { |name| File.join(root, name) } + Dir.glob(File.join(root, 'docs', '*.md'))
errors = []
files.each do |file|
  File.foreach(file).with_index(1) do |line, number|
    line.scan(/\[[^\]]*\]\((<[^>]+>|[^)]+)\)/).flatten.each do |target|
      target = target.delete_prefix('<').delete_suffix('>').split('#', 2).first.to_s
      next if target.empty? || target.match?(/\A[a-z][a-z0-9+.-]*:/i)
      path = File.expand_path(URI::DEFAULT_PARSER.unescape(target), File.dirname(file))
      errors << "#{file.delete_prefix(root + '/')}:#{number}: missing #{target}" unless File.exist?(path)
    end
  end
end
abort errors.join("\n") unless errors.empty?
puts "Documentation links valid (#{files.length} current documents; historical archives excluded)."
