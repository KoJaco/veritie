-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetAccountByStripeCustomerID :one
SELECT * FROM accounts WHERE stripe_customer_id = $1;
