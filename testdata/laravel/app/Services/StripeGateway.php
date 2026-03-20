<?php

namespace App\Services;

class StripeGateway implements PaymentGateway
{
    public function charge(int $amount): bool
    {
        return true;
    }

    public function refund(int $amount): bool
    {
        return true;
    }
}
