<?php

namespace App\Models;

class User
{
    public string $name;
    public string $email;

    public function posts(): array
    {
        return [];
    }

    public static function find(int $id): ?self
    {
        return null;
    }
}
