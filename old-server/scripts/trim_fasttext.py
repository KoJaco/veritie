#!/usr/bin/env python3
"""
Trim a Facebook fastText .bin to the first N words without loading it all.

Usage:
    python trim_fasttext.py \
        --in  ../models/fasttext/cc.en.300.bin \
        --out ../models/fasttext/cc.en.300.100k.bin \
        --vocab 100000
"""

import argparse, pathlib
from tqdm import tqdm
from gensim.models.fasttext import load_facebook_model

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--in",  required=True, dest="inp")
    ap.add_argument("--out", required=True, dest="out")
    ap.add_argument("--vocab", type=int, default=100_000)
    args = ap.parse_args()

    src = pathlib.Path(args.inp)
    assert src.is_file(), f"{src} missing"

    print(f"🔹 Loading (mmap) {src} …")
    ft = load_facebook_model(src.as_posix(), mmap='r')   # no full RAM load

    top = min(args.vocab, len(ft.wv))
    print(f"🔹 Keeping top {top:,} words")
    kv = ft.wv

    # Slice dictionary + matrix (vectors are mem-mapped, slice is cheap)
    kv.vectors = kv.vectors[:top].copy(order='C')   # materialise only slice
    kv.index_to_key = kv.index_to_key[:top]
    kv.key_to_index = {w:i for i,w in enumerate(kv.index_to_key)}

    print(f"🔹 Writing {args.out} …")
    kv.save_facebook_model(args.out)
    print("✅ Done   ->", pathlib.Path(args.out).stat().st_size/1e6, "MB")

if __name__ == "__main__":
    main()



# OR USE THE FASTTEXT CLI

# # build or install the fastText CLI once
# git clone https://github.com/facebookresearch/fastText.git
# cd fastText && mkdir build && cd build
# cmake .. && make -j$(nproc)
# sudo make install            # installs /usr/local/bin/fasttext


# mkdir tmp_dump

# # 1-a dictionary: plain text  (one word per line)
# fasttext dump cc.en.300.bin dict  > tmp_dump/dict.txt

# # 1-b input matrix: binary float32 rows
# fasttext dump cc.en.300.bin input > tmp_dump/input.vec


# Bash
# N=100000
# DIM=300
# ROW_BYTES=$((DIM * 4))            # 300 × 4 = 1200
# BYTE_COUNT=$((N * ROW_BYTES))     # 120,000,000

# # 2-a dictionary: top N lines  (header line is NOT present in .dict)
# head -n "$N" tmp_dump/dict.txt > dict_100k.txt

# # 2-b input matrix: first N binary rows
# head -c "$BYTE_COUNT" tmp_dump/input.vec > input_100k.vec

# Add dummy file, list each word once with a dummy frequency of "1"
# paste <(cut -f1 dict_100k.txt) <(yes 1 | head -n $N) > dummy.txt

# Re-pack into a Facebook binary
# fasttext skipgram \
#   -input dummy.txt \
#   -output cc.en.300.100k \
#   -dim 300 -epoch 1 -minCount 1 -bucket 1 -thread 1 -lr 0.01 -silent 1 \
#   -pretrainedVectors input_100k.vec


# Delete unwanted
# rm cc.en.300.100k.vec dummy.txt
# rm -r tmp_dump

# Size check
# ls -lh ../models/fasttext/cc.en.300.100k.bin   # ≈ 80 MB


# Convert to vec for use with embedding.Load
# 0. variables
# SRC=models/fasttext/cc.en.300.bin   # 7 GB original (or 547 MB trimmed)
# DST=models/fasttext/cc.en.300.100k.vec
# N=100000                            # number of words to keep

# # 1. dump vectors to text (≈ 10 GB tmp)
# fasttext dump $SRC output > full.vec

# # 2. slice header + N rows  (header is line-1)
# ((LINES=N+1))
# head -n "$LINES" full.vec > "$DST"
# rm full.vec                        # cleanup (optional)

# ls -lh "$DST"                      # ≈ 115 MB
