FROM golang:alpine AS build
# Create appuser.
ENV USER=appuser
ENV UID=10001 
# See https://stackoverflow.com/a/55757473/12429735RUN 
RUN adduser \    
    --disabled-password \    
    --gecos "" \    
    --home "/" \    
    --shell "/sbin/nologin" \    
    --no-create-home \    
    --uid "${UID}" \    
    "${USER}"

WORKDIR $GOPATH/src/github.com/timmydo/adometrics
COPY go.mod go.sum metrics.go ./
RUN go build -o /adometrics

FROM alpine
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
COPY --from=build /adometrics /
USER appuser:appuser
ENTRYPOINT ["/adometrics"]