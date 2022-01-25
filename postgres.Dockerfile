FROM postgres:14.1

RUN apt update && \
    apt install -y git pkg-config make gcc postgresql-server-dev-14 && \
    apt install liburiparser-dev liburiparser1 liburiparser-doc && \
    git clone https://github.com/petere/pguri.git

WORKDIR pguri

RUN make && \
    make install
