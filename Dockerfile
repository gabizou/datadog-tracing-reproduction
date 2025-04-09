# syntax=docker/dockerfile:1-labs

FROM golang:1.24-alpine AS base

RUN apk add --no-cache gcc musl-dev

FROM base as dependencies

WORKDIR /build

COPY --parents go.mod go.sum cmd pkg /build/

RUN go env -w \
	GOMODCACHE=/root/.cache/go-build \
	GOFLAGS="-tags=musl" \
	CGO_ENABLED=1

RUN --mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/root/go/pkg/mod \
	go mod tidy --diff

RUN --mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/root/go/pkg/mod \
	go mod download -x

# Build app binaries
FROM dependencies AS app-build

RUN --mount=type=bind,source=.git,target=/build/.git \
	--mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/root/go/pkg/mod \
	go build -o /out/worker \
	-installsuffix "static" \
	./cmd/worker && \
    go build -o /out/bootstrap \
    		-installsuffix "static" \
    		./cmd/starter

# Base runtime image
FROM alpine:3.21 AS base-app
# Create a non-root user
ARG APP_GROUP=stellar APP_USER=stellar APP_UID=65535 APP_GID=65535 APP_HOME=/app
RUN addgroup -g ${APP_GID} ${APP_GROUP} \
	&& adduser \
	--shell /sbin/nologin \
	--disabled-password \
	--no-create-home \
	--home ${APP_HOME} \
	--uid ${APP_UID} \
	--ingroup ${APP_GROUP} ${APP_USER}
USER ${APP_UID}:${APP_GID}

# Worker image

FROM base-app AS worker

COPY --from=app-build /out/worker /worker

ENTRYPOINT ["/worker"]

# Starter image

FROM base-app AS starter

COPY --from=app-build /out/starter /starter

ENTRYPOINT ["/starter"]
