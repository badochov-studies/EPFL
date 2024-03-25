TO_ZIP := processor build.sh go.mod main.go run.sh
ZIP_NAME ?= HW1.zip
EXE_NAME ?= OoO470
ASS_PATH ?= ../CS470-Homeworks/HW1/

all: build

build:
	./build.sh

zip:
	zip -r $(ZIP_NAME) $(TO_ZIP)

test: zip
	cp $(ZIP_NAME) $(ASS_PATH)
	cd $(ASS_PATH) && \
		unzip $(ZIP_NAME) && \
		docker run -v .:/home/root/cs470 cs470 bash -c "export PATH="$$PATH":/usr/local/go/bin && cd /home/root/cs470 && ./runall.sh && ./testall.sh"
	cd $(ASS_PATH) && rm -rf $(ZIP_NAME) $(TO_ZIP) $(EXE_NAME) && find . -name user_output.json -delete
	make clean_zip

clean_zip:
	rm $(ZIP_NAME)

clean: clean_zip
	rm -f $(EXE_NAME)