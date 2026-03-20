<?php

namespace App\Providers;

use App\Services\PaymentGateway;
use App\Services\StripeGateway;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(PaymentGateway::class, StripeGateway::class);
        $this->app->bind('mailer', \App\Services\CustomMailer::class);
    }
}
