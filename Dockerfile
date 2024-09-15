# syntax=docker/dockerfile:1
FROM golang
ADD . /src
WORKDIR /src

RUN go build -o /bin/scratchcord-server .

FROM scratch
COPY --from=0 /bin/scratchcord-server /bin/scratchcord-server
CMD ["/bin/scratchcord-server"]