<?php

/**
 * Get the application version.
 *
 * @return string
 */
function app_version(): string
{
    return '1.0.0';
}

/**
 * Generate a URL-friendly slug from a string.
 *
 * @param string $title
 * @param string $separator
 * @return string
 */
function str_slug(string $title, string $separator = '-'): string
{
    return strtolower(str_replace(' ', $separator, $title));
}
