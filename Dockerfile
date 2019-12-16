FROM golang:latest
RUN apt-get install git

RUN mkdir /app
ADD . /app/
WORKDIR /app

RUN git config --global user.email "system@dockercontainer"
RUN git config --global user.name "system"

RUN go install .

CMD /go/bin/g-wiki --wiki /data

EXPOSE 8000
