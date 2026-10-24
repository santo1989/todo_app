FROM cgr.dev/chainguard/go:latest-dev AS builder

WORKDIR /app

COPY go.mod ./
COPY . .
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/daily-notes-app .

FROM cgr.dev/chainguard/static:latest

WORKDIR /app
COPY --from=builder /out/daily-notes-app /daily-notes-app

EXPOSE 8080

ENV PORT=8080
ENV DB_PATH=/data/daily_notes.db

VOLUME ["/data"]
ENTRYPOINT ["/daily-notes-app"]
