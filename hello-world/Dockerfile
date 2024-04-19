FROM nexus.marketspring.co.kr/docker-hosted/grafana-loki-extension layer
FROM public.ecr.aws/lambda/nodejs:20
COPY --from=layer /opt /opt

COPY package.json .
RUN npm install
COPY . .

CMD ["index.handler"]