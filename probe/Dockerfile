FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.buildVersion=${VERSION}" -o /out/probe ./cmd/probe

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/probe /usr/local/bin/probe
EXPOSE 9464
ENTRYPOINT ["/usr/local/bin/probe"]
