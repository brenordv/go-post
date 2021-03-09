del go-post.exe
del go-hub.exe
del go-mongo.exe

go build -o go-post.exe gopost.go utils.go
go build -o go-hub.exe gohub.go utils.go
go build -o go-mongo.exe gomongo.go utils.go

7z a -tzip go-post--windows-amd64--%*.zip *.exe
