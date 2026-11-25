# Sandbox image for Fractory Runs on this repository: Go, tmux (the UI and session tests drive
# real tmux servers), git, gh and Claude Code.
FROM golang:1.27.4-bookworm

RUN apt-get update && apt-get install -y \
  git \
  curl \
  jq \
  tmux \
  && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
  | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
  && apt-get update && apt-get install -y gh \
  && rm -rf /var/lib/apt/lists/*

# sandcastle docker build-image passes the host UID/GID so bind-mounted files share an owner.
ARG AGENT_UID=1000
ARG AGENT_GID=1000
RUN groupadd -o -g $AGENT_GID agent && useradd -o -m -u $AGENT_UID -g $AGENT_GID -d /home/agent agent
USER ${AGENT_UID}:${AGENT_GID}

RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH="/home/agent/.local/bin:/usr/local/go/bin:/home/agent/go/bin:$PATH"
ENV GOPATH=/home/agent/go GOCACHE=/home/agent/.cache/go-build

WORKDIR /home/agent
ENTRYPOINT ["sleep", "infinity"]
