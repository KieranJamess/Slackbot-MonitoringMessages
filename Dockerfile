FROM golang:1.21

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

#Copy code
COPY main.go ./
COPY config.json ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /slackbot

# Networking - Change port to match the port in main.go
EXPOSE 80

# Run
CMD ["/slackbot"]

## To use, "docker build -t <image_name> ." + docker run -p 80:80 <image_name>