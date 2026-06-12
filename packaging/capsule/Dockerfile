FROM golang:1.25.5-bookworm AS liza-builder

WORKDIR /src
COPY . .
RUN rm -rf internal/embedded/contracts internal/embedded/skills internal/embedded/support-docs internal/embedded/docs internal/embedded/specs \
  && mkdir -p internal/embedded/contracts internal/embedded/skills internal/embedded/support-docs \
  && cp contracts/*.md internal/embedded/contracts/ \
  && cp -r skills/* internal/embedded/skills/ \
  && cp support-docs/*.md internal/embedded/support-docs/ \
  && find internal/embedded/skills/ -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
RUN CGO_ENABLED=0 go build -o /out/liza ./cmd/liza
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    make \
  && rm -rf /var/lib/apt/lists/*
# NOTE: Reproducibility risk - install scripts from main branch without commit pinning
# For production Daytona snapshots, consider pinning specific commit SHAs
RUN curl -fsSL https://raw.githubusercontent.com/liza-mas/stacklit-cli/main/install.sh | INSTALL_DIR=/out sh
RUN curl -fsSL https://raw.githubusercontent.com/liza-mas/scip-search/main/install.sh | BRANCH=main INSTALL_DIR=/out bash
RUN go install github.com/scip-code/scip-go/cmd/scip-go@latest \
  && cp /go/bin/scip-go /out/scip-go

FROM ubuntu:24.04

ARG DEBIAN_FRONTEND=noninteractive
ARG YQ_VERSION=v4.49.1

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    fd-find \
    git \
    gh \
    jq \
    nodejs \
    npm \
    python3 \
    python3-venv \
    ripgrep \
    unzip \
    zsh \
  && rm -rf /var/lib/apt/lists/*
RUN ln -sf /usr/bin/fdfind /usr/local/bin/fd
RUN arch="$(dpkg --print-architecture)" \
  && case "$arch" in \
    amd64) yq_arch=amd64 ;; \
    arm64) yq_arch=arm64 ;; \
    *) echo "unsupported yq architecture: $arch" >&2; exit 1 ;; \
  esac \
  && curl -fsSL "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_${yq_arch}" -o /usr/local/bin/yq \
  && chmod +x /usr/local/bin/yq

RUN useradd -m -s /bin/bash liza

# NOTE: Reproducibility risk - npm packages without pinned versions
# For production Daytona snapshots, consider pinning specific package versions
RUN npm install -g \
    @ast-grep/cli \
    @openai/codex \
    @sourcegraph/scip-python \
    @sourcegraph/scip-typescript \
    opencode-ai
# NOTE: Reproducibility risk - install script without commit pinning
# For production Daytona snapshots, consider pinning specific commit SHA
RUN curl -LsSf https://astral.sh/uv/install.sh | sh \
  && UV_TOOL_DIR=/opt/uv/tools UV_TOOL_BIN_DIR=/usr/local/bin /root/.local/bin/uv tool install semble \
  && chmod -R a+rX /opt/uv \
  && command -v semble >/dev/null
# NOTE: Reproducibility risk - install script without commit pinning
# For production Daytona snapshots, consider pinning specific commit SHA
RUN curl -fsSL https://claude.ai/install.sh | bash \
  && if [ -x /root/.local/bin/claude ]; then cp /root/.local/bin/claude /usr/local/bin/claude; fi \
  && command -v claude >/dev/null

COPY --from=liza-builder /out/liza /usr/local/bin/liza
COPY --from=liza-builder /out/stacklit /usr/local/bin/stacklit
COPY --from=liza-builder /out/scip-search /usr/local/bin/scip-search
COPY --from=liza-builder /out/scip-go /usr/local/bin/scip-go
COPY --from=liza-builder /usr/local/go /usr/local/go

USER liza
WORKDIR /workspace

ENV PATH=/usr/local/go/bin:$PATH
ENV OPENCODE_CONFIG=/home/liza/.config/opencode/opencode.json
ENV OPENCODE_CONFIG_DIR=/home/liza/.config/opencode
ENV OPENCODE_DATA_DIR=/home/liza/.local/share/opencode

CMD ["liza", "tui"]
