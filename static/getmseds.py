#!/usr/bin/env python3

import requests

# https://stackoverflow.com/a/16696317 thx
def download_file(url, local_filename):
    # NOTE the stream=True parameter
    r = requests.get(url, stream=True)
    with open(local_filename, 'wb') as f:
        for chunk in r.iter_content(chunk_size=1024):
            if chunk: # filter out keep-alive new chunks
                f.write(chunk)
                #f.flush() commented by recommendation from J.F.Sebastian
    return local_filename

r = requests.get("https://seedhelper.figgyc.uk/static/mseds/list")
for line in r.text.splitlines():
    download_file("https://seedhelper.figgyc.uk/static/mseds/" + line, line)