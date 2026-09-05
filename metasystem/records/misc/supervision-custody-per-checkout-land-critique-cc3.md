# Supervision custody landing member: code review, third round (chain scp-build1-cc3)

Reviewed tree 9b893a7e218f29662bb8fa6b509157f5ff3fc11d (chain scp-build1, round 3). Critic: Claude Fable 5.1. One material finding, in the fixture self-check, not the custody code; the member goal's review rounds are spent.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SCC-61 | material; risk decision to Wido | The self-check's announcement scan treats every JSON file under a harness root's mains directory as a main announcement; the protocol cursor and the reaped-after-claim stamp carry no pid, so the scan can misjudge them. The custody code and the invariant test are not affected. | Either a fourth member round, or accept the risk and schedule a tier-1 fixture item. |
| SCC-62 | noted | The operator-layout scenario's harness root is one level below its state root, so the scan looks in a mains directory that does not exist; the arm-script audit still covers that main. | with the fixture item |
| SCC-63 | noted | Cross-checkout rows are dropped and listed but no production caller reads the list: drop-and-silence. | with a registry item |
| SCC-64 | noted | The auditor wrapper adds a shell start per hook call under the hook's four-second deadline; cannot be separated from load. | none |
| SCC-65 | noted | The ancestor walk under set -e can flake if an ancestor exits mid-walk. | with the fixture item |
