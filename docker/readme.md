
# Construire l’image depuis docker/Dockerfile
docker build . -t fgtech.fr/ttyd:latest -f docker/Dockerfile .

# Ouvrir un shell dans le conteneur
docker run --rm -it fgtech.fr/ttyd:latest /bin/bash
