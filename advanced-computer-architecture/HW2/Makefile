TO_ZIP := scheduler build.sh go.mod main.go run.sh README.md LICENSE
ZIP_NAME ?= HW2.zip
EXE_NAME ?= VLIW470
ASS_PATH ?= ../CS470-Homeworks/HW2/

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
	cd $(ASS_PATH) && rm -rf $(ZIP_NAME) $(TO_ZIP) $(EXE_NAME) && find . -type f \( -name "simple.json" -name "pip.json" \) -delete
	make clean_zip

clean_zip:
	rm $(ZIP_NAME)

clean:
	-rm -f $(EXE_NAME)
	make clean_zip