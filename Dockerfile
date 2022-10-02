FROM python:latest as build

WORKDIR /tmp
RUN git clone https://github.com/Silicondust/libhdhomerun.git && \
    cd libhdhomerun && \
    make

FROM python:latest 

RUN git clone https://github.com/kincl/hdhr-legacy-proxy.git

WORKDIR hdhr-legacy-proxy

RUN pip install -r requirements.txt && \
    pip install git+https://github.com/kincl/hdhr.git
COPY --from=build /tmp/libhdhomerun/libhdhomerun.so .
ENV LD_LIBRARY_PATH /hdhr-legacy-proxy

CMD ["gunicorn", "hdhr_legacy_proxy"]
