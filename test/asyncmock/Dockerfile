FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder

WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /async-test-mock main.go

FROM scratch
COPY --from=builder /async-test-mock /async-test-mock
EXPOSE 8080
ENTRYPOINT ["/async-test-mock"]
