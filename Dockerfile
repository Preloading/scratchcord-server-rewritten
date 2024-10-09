# syntax=docker/dockerfile:1
FROM golang AS build-stage
ADD . /src
WORKDIR /src

RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/scratchcord-server .

FROM gcr.io/distroless/base-debian12 AS build-release-stage
COPY --from=build-stage /bin/scratchcord-server /bin/scratchcord-server
WORKDIR /config

ENV SCRATCHCORD_DB_PATH="/config/sqlite/scratchcord.db" \
    SCRATCHCORD_MEDIA_PATH="/config/uploads" \
    SCRATCHCORD_KEY_PATH="/config/keys" \
    SCRATCHCORD_ADMIN_PASSWORD="scratchcord" \
    SCRATCHCORD_SERVER_URL="http://127.0.0.1:3000" \
    SCRATCHCORD_ADMIN_PASSWORD="scratchcord"

CMD ["/bin/scratchcord-server"]