FROM alpine
WORKDIR /src
RUN apk add --no-cache go
COPY ./ /src
RUN go build .
ENTRYPOINT [ "./veto" ]
