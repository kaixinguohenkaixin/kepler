
import os


def search(path):
    for filename in os.listdir(path):
        fp = os.path.join(path, filename)
        if os.path.isfile(fp) and want_file(fp) and check_ext(fp):
            check_file(fp)
        elif os.path.isdir(fp):
            search(fp)

# filter file in some directory
def want_file(fp):
    tags =["build", "vendor"]
    for tag in tags:
        if tag in fp:
            return False
    return True

# only search some file with specific
def check_ext(fp):
    exts =[".go", ".yml"]
    # exts =[".alltools", "Dockerfile"]
    for ext in exts:
        if ext in fp:
            return True
    return False

def check_file(path):
    # with open(path) as file:
    file = open(path)
    text = file.read().decode('utf-8')
    lines = text.split('\n')
    for line in lines:
        if "//" not in line:
            if "ethereum" in line or "Ethereum" in line:
                print "%s: %s" % (path, line)

search("./")