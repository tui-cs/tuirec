# TUIcast: AI-Powered Terminal GIF Generation for Terminal.Gui Developers

**TUIcast lets Terminal.Gui developers generate polished, animated GIF recordings of their TUI apps by simply describing what they want to demonstrate.**

---

Seattle, WA – Today, Terminal.Gui developers can finally say goodbye to manually scripting screen recordings and wrestling with terminal capture tools. TUIcast is a new service that takes a natural language goal – "show how popover menus work" – and produces a ready-to-tweet GIF of an AI navigating your actual Terminal.Gui application.

Tig Kindel, Terminal.Gui maintainer, said: "Every time I want to show off a new feature, I spend an hour fiddling with recording tools, writing keystroke macros, and tweaking GIF settings. TUIcast turns that into a single sentence."

TUIcast works by spinning up a real PTY environment running your Terminal.Gui application, using Claude to drive keystrokes and mouse interactions toward the stated goal, recording the session with asciinema, and rendering a polished GIF with configurable font and theme. The developer gets a download link. No local setup required.

TUIcast is available today as a web service. Developers provide their app's binary or a GitHub repo reference, describe what they want demonstrated, and receive a GIF within minutes.

---

## FAQ

**Q: What TUI frameworks does it support?**
A: Terminal.Gui is the primary target. Any .NET console app that runs over a PTY will work. Other frameworks (Spectre.Console, etc.) are untested but likely functional.

**Q: How does the AI know what to do?**
A: You provide a goal in plain English. Claude is given the goal, observes the terminal screen state after each action, and decides the next keystroke or mouse event. It iterates until the goal appears complete or a timeout is hit. You can also provide a deterministic keystroke script if you want exact control.

**Q: What if the AI does something wrong or gets stuck?**
A: You get a GIF either way. If the result isn't right, you refine the goal description and re-run. Iteration is fast – typically under two minutes per run.

**Q: Can I control the look of the GIF?**
A: Yes. Terminal size, font, color theme, and playback speed are all configurable. Defaults are tuned for legibility in social media embeds.

**Q: Does my app need to be publicly available?**
A: No. You can push a Docker image to Azure Container Registry and reference it by name, or provide a self-contained binary.

**Q: How is it hosted?**
A: Each recording job runs in an isolated Azure Container Instance. Jobs are queued via Azure Service Bus. GIFs are stored in Azure Blob Storage and served via a short-lived URL.

**Q: Is this secure?**
A: Jobs run in isolated, ephemeral containers with no network egress. Containers are destroyed after the job completes.

**Q: What are the current limitations?**
A: Apps requiring external network calls, database connections, or complex setup beyond a single binary are not yet supported. Maximum recording duration is 60 seconds.

**Q: What does it cost?**
A: Currently free for Terminal.Gui maintainers and contributors. Broader pricing TBD.

**Q: What tech will we need to buy, license, use, or invent?**
A: node-pty (MIT), xterm.js 5.x (MIT), asciinema (AGPL – review needed), agg (MIT), Azure Container Instances, Azure Service Bus, Azure Blob Storage, Anthropic Claude API. No novel inventions required; integration is the work.

**Q: What are the riskiest parts?**
A: AI reliability is the primary risk – Claude may navigate incorrectly or non-deterministically for complex interactions. Mitigation is the hybrid approach: scripted playback for known demos, AI vision loop for exploratory ones. Secondary risk is asciinema's AGPL license; agg (the GIF renderer) is MIT clean.

**Q: How will we measure success?**
A: GIFs generated per week; re-run rate (lower is better); developer-reported time saved vs. manual recording; social engagement on GIFs published from TUIcast.
