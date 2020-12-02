FROM golang:alpine AS build

WORKDIR /src/
COPY . /src/

RUN go get -d -v ./...
RUN CGO_ENABLED=0 go build -o /bin/app

FROM scratch
COPY --from=build /bin/app /bin/app
ENTRYPOINT ["/bin/app"]