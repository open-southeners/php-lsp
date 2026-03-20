<?php

namespace App\Models;

class Category
{
    public string $name;
    public string $slug;

    public function products(): array
    {
        return [];
    }
}
