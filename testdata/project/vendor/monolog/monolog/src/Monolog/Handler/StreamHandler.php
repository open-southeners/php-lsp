<?php
namespace Monolog\Handler;

use Monolog\Logger;

/**
 * Stores logs to a file.
 */
class StreamHandler
{
    protected string $stream;

    /**
     * Get the associated logger.
     *
     * @return Logger The logger instance
     */
    public function getLogger(): Logger
    {
        return $this->logger;
    }

    /**
     * Handle a log record.
     *
     * @param array $record The log record
     * @return bool Whether the record was handled
     */
    public function handle(array $record): bool
    {
        return true;
    }
}
