FROM golang:latest

RUN apt-get update && \
    apt-get install -y nodejs npm && \
    npm install -g @stoplight/spectral-cli && \
    apt-get clean && rm -rf /var/lib/apt/lists/* \
    
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main ./cmd/main.go

EXPOSE 1337

CMD ["./main"]