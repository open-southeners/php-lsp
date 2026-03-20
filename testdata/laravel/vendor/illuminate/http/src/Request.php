<?php

namespace Illuminate\Http;

class Request
{
    public function input(string $key, mixed $default = null): mixed
    {
        return $default;
    }

    public function all(): array
    {
        return [];
    }

    public function query(string $key = null): mixed
    {
        return null;
    }

    public function header(string $key): ?string
    {
        return null;
    }
}
