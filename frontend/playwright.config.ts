import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  use: {
    baseURL: process.env.APP_ORIGIN || "http://localhost:8080",
    headless: true,
  },
  reporter: "list",
  workers: 1,
});
