import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
  outputFileTracingRoot: path.join(__dirname),
  // Дев-сервер по умолчанию блокирует кросс-origin запросы к своим ассетам
  // (_next/*, HMR-вебсокет) — 403, если открывать его не с localhost, а по
  // Tailscale-адресу с другого устройства.
  allowedDevOrigins: ["100.64.229.103", "192.168.3.103"],
};

export default nextConfig;
