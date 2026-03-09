<?php
namespace App;

use Monolog\Logger;
use Monolog\Handler\StreamHandler;

class Service
{
    private Logger $logger;

    public function __construct(Logger $logger)
    {
        $this->logger = $logger;
    }

    public function run(): void
    {
        $this->logger->info('hello');
        $handler = new StreamHandler();
        $handler->getLogger()->info('via handler');
        $handler->handle(['message' => 'test']);
        Logger::create('app');
        self::helper();
    }

    private static function helper(): void {}
}
