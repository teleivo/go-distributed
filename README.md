# go-distributed

This repo is a place for me to improve my [go](https://golang.org) and learn and apply
concepts and patterns used in [distributed systems](https://en.wikipedia.org/wiki/Distributed_computing).

# Limitations

The code in this repository is merely for my own educational purpose. If it is
useful to you even better ☺️. please create an issue if you would like to collaborate on a
topic and learn together.

⚠️ I would advise against using any of this code in production!⚠️
There are plenty of production grade libraries out there that will fit your use
case.

# Rate Limiting

## Token Bucket

I implemented the [token bucket](https://en.wikipedia.org/wiki/Token_bucket)
algorithm in [./ratelimit/](./ratelimit). It is currently only a very naive an
in-memory implementation. It enforces the rate limit only per endpoint. So it
does not enforce a single rate limit for multiple endoints and per user (IP
address, API token, ...).
