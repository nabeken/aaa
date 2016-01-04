dist: npm_install main
	zip -r lambda.zip main node_modules *.js aaa_lambda.toml

npm_install:
	npm install

main:
	GOOS=linux go build -o main

distclean: clean
	rm -f lambda.zip

clean:
	rm -rf main
