agent-running.jsonl — 9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:126-127.
agent-completed-user-delivery.jsonl — 9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:126-127,220-222.
agent-completed-attachment-delivery.jsonl — 9b4a48c5-e0ce-46bb-99b1-ca3644a08ec4.jsonl:48-49,100-102.
notification-without-enqueue.jsonl — 9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:126-127,222; enqueue line 220 removed.
repeated-notification-one-id.jsonl — 9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:126-127,220,222.
resumed-then-completed.jsonl — 36c93128-ec16-4146-8def-3f706ba4ef10.jsonl:13872-13873,14020,14039-14040,14069.
unknown-status.jsonl — constructed from 9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:126-127,220 by replacing observed `completed` with unobserved `paused`; the observed histogram has no unknown word.
bash-running.jsonl — c53cbef5-7be0-48fa-83f1-876f40a3c193.jsonl:176,185.
bash-completed.jsonl — c53cbef5-7be0-48fa-83f1-876f40a3c193.jsonl:176,185,188.
sync-agent-not-a-task.jsonl — 9b4a48c5-e0ce-46bb-99b1-ca3644a08ec4.jsonl:21120-21121.
agent-killed.jsonl — 2c27f5cc-245a-4735-a402-dafc351e4caf.jsonl:84-85,135.
bash-failed.jsonl — c53cbef5-7be0-48fa-83f1-876f40a3c193.jsonl:780-781,839.
agent-killed-then-resumed.jsonl — constructed by combining 2c27f5cc-245a-4735-a402-dafc351e4caf.jsonl:84-85,135 with the successful resume shape at 36c93128-ec16-4146-8def-3f706ba4ef10.jsonl:14039-14040 and changing its task id to the killed task.
agent-stopped.jsonl — constructed by combining the Agent launch at 2c27f5cc-245a-4735-a402-dafc351e4caf.jsonl:84-85 with the observed `stopped` word at 9b4a48c5-e0ce-46bb-99b1-ca3644a08ec4.jsonl:28259; the observed stopped summary has no tool-use id and cannot join.
agent-async-without-flag.jsonl — 9b7e334d-919d-47e9-9555-a9f2ed557072.jsonl:105-106.
