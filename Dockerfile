FROM golang:1.12.3 AS BUILD

#doing dependency build separated from source build optimizes time for developer, but is not required
#install external dependencies first
# ADD go-plugins-helpers/Gopkg.toml $GOPATH/src/go-plugins-helpers/
# ADD /schellyhook.go $GOPATH/src/schellyhook/schellyhook.go
# RUN go get -v schellyhook

# #now build source code
# ADD schellyhook $GOPATH/src/schellyhook
# RUN go get -v schellyhook

# ADD schellyhook-sample $GOPATH/src/schellyhook-sample
# RUN go get -v schellyhook-sample

RUN mkdir /schellyhook
WORKDIR /schellyhook

ADD go.mod .
ADD go.sum .
RUN go mod download

#now build source code
ADD schellyhook/ ./
ADD schellyhook-sample/ ./schellyhook-sample
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /go/bin/schellyhook .
# RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /go/bin/schellyhook-sample ./schellyhook-sample

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
