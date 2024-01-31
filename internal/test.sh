
cat << EOF
[url "ssh://git@github.com/"]
    insteadOf = https://github.com/
EOF >> ~/.gitconfig
go test ./...
