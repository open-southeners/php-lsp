<?php

/**
 * Create a collection from the given value.
 *
 * @param mixed $value
 * @return \Illuminate\Support\Collection
 */
function collect($value = []): \Illuminate\Support\Collection
{
    return new \Illuminate\Support\Collection($value);
}

/**
 * Get / set the specified configuration value.
 *
 * @param string|null $key
 * @param mixed $default
 * @return mixed
 */
function config(?string $key = null, $default = null)
{
    return $key;
}
