FROM mcr.microsoft.com/playwright:v1.49.1-noble

WORKDIR /app

COPY package.json ./
RUN npm install --no-audit --no-fund

COPY . .

# Default command runs all tests; override with args for subset runs
CMD ["npx", "playwright", "test", "--reporter=list"]
