.PHONY: fetch
fetch:
	go run cmd/fetch/main.go -req ./cmd/fetch/samples/input.txt -resp ./cmd/fetch/samples/output.txt localhost:8080

.PHONY: quackquack
quackquack:
	go run cmd/quack/main.go -port 8080 -vh_config ./virtual_hosts.yaml -docroot ./docroot_dirs

.PHONY: submission
submission:
	go mod tidy
	rm -f submission.zip
	zip -r submission.zip . -x /.git/*