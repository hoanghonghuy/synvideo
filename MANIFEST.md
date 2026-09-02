# SynVideo PM Scaffold

This scaffold is installed on the SynVideo `develop` branch and defines the PM / AI Developer / Team Lead operating model used by current development.

Contents include:
- lightweight `AGENTS.md` router;
- focused agent skills for continuation, task execution, wave planning, code review, open-source research and product audit;
- remote-first control-plane authority/freshness protocol;
- product source-of-truth baseline;
- PM/AI Developer/Team Lead workflow and context policy;
- atomic remote task claiming + dedicated worktree isolation;
- TDD and exact-head Team Lead review constraints;
- duplicate-task prevention and abandoned-claim recovery;
- open-source research map;
- task board/template and accepted product decisions;
- Vue frontend, Go API and local infrastructure.

Branch policy:
- `main`: stable/release-ready history and may intentionally lag active development;
- `develop`: current integration/control-plane baseline;
- implementation: dedicated canonical task branches/worktrees with PRs to `develop`.

For shared workflow decisions, local Git state is not authoritative. Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` and inspect live GitHub plus explicit current `develop`/task/PR refs.
