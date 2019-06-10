FROM golang:1.12.3 AS BUILD

RUN mkdir /schellyhook
WORKDIR /schellyhook

ADD go.mod .
ADD go.sum .
RUN go mod download

#now build source code
ADD schellyhook/ ./schellyhook
ADD schellyhook-sample/ ./schellyhook-sample
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /go/bin/schellyhook ./schellyhook
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /go/bin/schellyhook-sample ./schellyhook-sample

FROM golang:1.12.3

VOLUME [ "/backup-source" ]
VOLUME [ "/backup-repo" ]

EXPOSE 7070

ENV LISTEN_PORT 7070
ENV LISTEN_IP '0.0.0.0'
ENV LOG_LEVEL 'debug'
ENV PRE_BACKUP_COMMAND ''
ENV POST_BACKUP_COMMAND ''
ENV PRE_POST_TIMEOUT_SECONDS '3600'

COPY --from=BUILD /go/bin/* /bin/
ADD startup.sh /
ADD pre-backup.sh /
ADD post-backup.sh /

CMD [ "/startup.sh" ]
