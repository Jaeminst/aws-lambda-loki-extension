FROM grafana-loki-extension layer
FROM public.ecr.aws/lambda/nodejs:20

COPY package.json .
RUN npm install
COPY . .

CMD ["index.handler"]

# Copy log extension
COPY --from=layer /opt /opt