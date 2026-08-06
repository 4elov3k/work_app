import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Управление счетами и актами",
  description: "Система управления счетами и актами для ИП Мыленкова Л.В.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru" suppressHydrationWarning>
      <body className="font-sans antialiased min-h-screen bg-background">
        {children}
      </body>
    </html>
  );
}
