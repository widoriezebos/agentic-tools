# fleet-channel-gateway

- State: queued
- Intent: One bot for the whole fleet, no lost messages (Wido's decision 2026-09-03, Design A): a single CHANNEL GATEWAY reads and posts on behalf of every machine. Telegram fixes one fact - a bot token has exactly one reader - so exactly one machine (the host m1, always on) holds the gateway under a LEASE that fails over if it dies; every inbound human message is written by the gateway into a shared git-synced INBOX on the transport, tagged with the ask id it answers; every machine reads its asks and replies from the inbox, never from the provider. Posting also routes through the gateway so a provider swap (Telegram to Slack) touches one machine and one config. DONE means: a machine with no provider token can raise an ask, get Wido's reply from the inbox, and post status through the gateway; the gateway lease fails over in a test; a cursor replay never duplicates an answer (keyed by message id); m0 and m0b no longer compete for one token.
- Origin: main
- Next step: INTENT: one gateway, one inbox, provider-agnostic fleet. CONSTRAINTS: exactly one reader per provider token (Telegram's rule); the gateway is the single trust boundary - Wido's identity check (per-provider user id + the shared TOTP secret) runs there once, downstream trusts the inbox; idempotent delivery keyed by provider message id; lease failover tested; the inbox rotates; the gateway host needs transport push rights. DESIGNED IN FROM THE START: N destinations (Telegram AND Slack in parallel) with a per-message-class ROUTING POLICY (config only, e.g. status to Slack, urgent asks to both), one reader per provider all writing the same inbox, FIRST VALID ANSWER WINS per ask id with the later one recorded and ignored. FREEDOMS: inbox format/location on the transport; the lease mechanism (reuse existing lease machinery); whether the gateway is a steward phase or its own runner. Tier 3 (R-54-m1): design, one review, one fold, one closing review, build, one code review. Companion: answer-archive (the aggregate answer log). Budget Wido's word at approval.
- OpenedAt: 2026-09-03T15:27:54Z
- Revision: 1

History:
- 2026-09-03T15:27:54Z 3K5AD6XK8Z3V6BQ4MADHFJ1856-m0-c5dbf036 open actor=human:Wido targets=fleet-channel-gateway
Integrity: sha256=d39c335a76d56215f3812ffb15e316e17d9bc8700dc9327b7e7a154ac7eaa22d
