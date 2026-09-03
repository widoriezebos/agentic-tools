# fleet-channel-gateway

- State: queued
- Intent: One bot for the whole fleet, no lost messages (Wido's decision 2026-09-03, Design A): a single CHANNEL GATEWAY reads and posts on behalf of every machine. Telegram fixes one fact - a bot token has exactly one reader - so exactly one machine (the host m1, always on) holds the gateway under a LEASE that fails over if it dies; every inbound human message is written by the gateway into a shared git-synced INBOX on the transport, tagged with the ask id it answers; every machine reads its asks and replies from the inbox, never from the provider. Posting also routes through the gateway so a provider swap (Telegram to Slack) touches one machine and one config. DONE means: a machine with no provider token can raise an ask, get Wido's reply from the inbox, and post status through the gateway; the gateway lease fails over in a test; a cursor replay never duplicates an answer (keyed by message id); m0 and m0b no longer compete for one token.
- Origin: main
- Next step: INTENT: one bot, one git inbox, FIRST COME FIRST SERVED - no leases (Wido 2026-09-03). THE RULE, every provider: receive -> commit to the shared git inbox -> confirm. Whichever machine commits a message first wins; the others find it already committed (idempotent by provider message id) and skip; git's push race is the arbiter, no lease or failover machinery anywhere. Telegram: all machines poll getUpdates WITHOUT an offset so all see the same pending messages; confirm the offset ONLY after the commit is durable (confirming consumes for everyone; Telegram drops unconfirmed after ~24h, harmless once committed). Slack: it delivers each event to one connected machine - that machine commits; same rule. A receiver that dies before committing has confirmed nothing, so the message is redelivered. Posting: any machine posts directly, to N destinations in parallel under a per-message-class routing policy; first valid answer wins per ask id. Identity check (per-provider user id + shared TOTP) runs on the committing machine. FREEDOMS: inbox format/location; polling interval. Tier 3 ladder. Companion: answer-archive. Budget Wido's word at approval.
- OpenedAt: 2026-09-03T15:27:54Z
- Revision: 3

History:
- 2026-09-03T15:27:54Z 3K5AD6XK8Z3V6BQ4MADHFJ1856-m0-c5dbf036 open actor=human:Wido targets=fleet-channel-gateway
- 2026-09-03T15:28:58Z N8ZXSVKRH5PMKV61FTRV64PZYS-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=fleet-channel-gateway
- 2026-09-03T15:29:29Z B4PRVCNJME0KDKFB5F1ZAZ9YFA-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=fleet-channel-gateway
Integrity: sha256=19acf568de1cb1911418f33a0feea28679844a234a367f1e0b8a22e038d08c8f
