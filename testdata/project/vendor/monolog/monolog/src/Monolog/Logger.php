<?php
namespace Monolog;

/**
 * Monolog log channel.
 */
class Logger
{
    /**
     * Adds a log record at INFO level.
     *
     * @param string $message The log message
     * @param array $context The log context
     * @return bool Whether the record has been processed
     */
    public function info(string $message, array $context = []): bool
    {
        return true;
    }

    /**
     * Adds a log record at ERROR level.
     *
     * @param string $message The log message
     * @return bool Whether the record has been processed
     */
    public function error(string $message): bool
    {
        return true;
    }

    public static function create(string $name): self
    {
        return new self();
    }
}
