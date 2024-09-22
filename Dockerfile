# syntax=docker/dockerfile:1
FROM golang AS build-stage
ADD . /src
WORKDIR /src

RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/scratchcord-server .

FROM gcr.io/distroless/base-debian12 AS build-release-stage
COPY --from=build-stage /bin/scratchcord-server /bin/scratchcord-server
WORKDIR /config

ENV SCRATCHCORD_DB_PATH="/config/sqlite/scratchcord.db"

CMD ["/bin/scratchcord-server"]