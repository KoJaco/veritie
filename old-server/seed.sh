cat > seed.sh <<'SH'
#!/bin/sh
set -e
apk add --no-cache curl
mkdir -p /data/models
echo "Fetching $TGZ_URL ..."
curl -L "$TGZ_URL" -o /data/models.tgz
tar -xzf /data/models.tgz -C /data/models
rm -f /data/models.tgz
find /data/models -type d -exec chmod 755 {} \;
find /data/models -type f -exec chmod 644 {} \;
echo "== df -h /data =="; df -h /data
echo "== du -h /data/models -d1 =="; du -h /data/models -d1 || true
echo "== ls -al /data/models =="; ls -al /data/models
echo "== ls -al /data/models/phi_roberta_onnx_int8 =="; ls -al /data/models/phi_roberta_onnx_int8 | sed -n '1,80p'
sleep 3600
SH
chmod +x seed.sh
