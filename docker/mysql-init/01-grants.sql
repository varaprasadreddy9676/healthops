-- Grant healthops user access to performance_schema for monitoring
GRANT SELECT ON performance_schema.* TO 'healthops'@'%';
FLUSH PRIVILEGES;
