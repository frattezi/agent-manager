
**Agent Automation**:
An Automation whose work is given to an agent as a prompt. The agent works alone and nobody
talks to it while it runs.
_Avoid_: Chat automation, prompt job, conversation

**Debug Session**:
A session the owner starts from a finished Automation Run to look into what it did. It runs
where they are, not where the Automation ran, and it begins knowing which Automation produced
the run it is asked about.
_Avoid_: Handoff, retry, attach

**Script Automation**:
An Automation whose work is a command: it runs, it ends, and what it printed stays readable
afterwards.
_Avoid_: Shell job, command task, bash hook

**Mode**:
The level of human oversight the work requires, either Work or Personal. Fractory fixes a
repository's Mode; an Automation carries its own, because one repository can hold both Work
and Personal Automations.
_Avoid_: Profile, environment, account
