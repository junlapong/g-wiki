FROM golang:latest
RUN apt-get install git

RUN mkdir /app
ADD . /app/
WORKDIR /app

RUN mkdir -p files
RUN git init files

RUN git config --global user.email "system@dockercontainer"
RUN git config --global user.name "system"

RUN go install .

CMD /go/bin/g-wiki

EXPOSE 8000
