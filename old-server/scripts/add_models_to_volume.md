## What is this?

I've added my models inside the supabase project 'schma', inside a bucket 'schma-models' or something like that.

Just have to ssh into the machine after deploying it and then re-download the models into the correct dirs.

```bash
fly ssh console --app schma-api

# then inside the machine
apk add --no-cache curl # if not already installed
mkdir -p /data/models/bge /data/models/fasttext # these should already exist

echo "Downloading ONNX model…"
curl -L "<SUPABASE_SIGNED_URL_FOR_COPIED_FROM_BROWSER>/model.int8.onnx" -o /data/models/bge/model.int8.onnx

# I've just copied over everything else inside the bge dir (except the unquantized model, which I've taken out... excluded all the rest of the curl statements for brevity.. manually do this).

echo "Downloading FastText…"
curl -L "<SUPABASE_SIGNED_URL_FOR_COPIED_FROM_BROWSER>/cc.en.300.100k.vec" -o /data/models/fasttext/cc.en.300.100k.vec

# confirm everything + also check the correct file sizes yeah
ls -lh /data/models/bge /data/models/fasttext
```
