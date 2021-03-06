# g-wiki

![test](https://github.com/junlapong/g-wiki/workflows/test/badge.svg)

A simple wiki built on golang with git as the storage back-end. Content
is formatted in [markdown syntax](http://daringfireball.net/projects/markdown/syntax).

The wiki is rendered with go templates and [bootstrap](http://getbootstrap.com) css.

The *g-wiki* is originally forked off from the wonderfully simple [original *g-wiki*](https://github.com/aspic/g-wiki),
then heavily modified and customized for my personal use and needs,
still _KISS_-ing on the way.

## Build and run locally

Ensure that go is installed. Download dependencies and compile the binary by:

```
go get -u github.com/junlapong/g-wiki
```

Create a git repository in some folder, for example `files`

```
git init files
```

You can now run g-wiki with the standard settings by executing the binary

```
./g-wiki
```

Point your web browser to [http://localhost:8000](http://localhost:8000) to see the wiki in action.
The wiki will try to store files in the `files` folder if configured as above.
This folder has to exist and be writeable by the user running the g-wiki
instance.

## Docker

Ensure that docker is installed. The docker file will create a `files` directory for you, and initialize a git repository there. Rembember that these files are not persistent. Dependent on your environement run docker as root (or not) and execute the following commands:

```
docker build -t go-wiki:latest .
```

If this executes succesfully your container is ready:

```
docker run -d -p 8000:8000 go-wiki:latest
```

This starts the web application in deamon mode, and the application should be accessible on [http://localhost:8000](http://localhost:8000)


## TODO

- [ ] Embedded template to build bainary, maybe [go.rice](https://github.com/GeertJohan/go.rice). [see](https://github.com/nohal/g-wiki#develop)
- [ ] fix FIXMEs (sanitization of paths, etc.)
- [ ] allow moving (renaming) nodes in the repo
- [ ] allow deleting files from repo
- [ ] use pure Go git implementation, maybe [go-git](https://github.com/go-git/go-git)
- [ ] [LATER] nice JS editor, with preview of markdown... but how to ensure compat. with blackfriday? or, VFMD everywhere?..

## References 

- https://github.com/aspic/g-wiki
- https://github.com/aspotashev/g-wiki
- https://github.com/nohal/g-wiki

## Inspire

- https://github.com/SpencerCDixon/exocortex
- https://wiki.js.org
