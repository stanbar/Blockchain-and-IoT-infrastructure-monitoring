import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "SmartGrid-BC Demo",
  description: "Verifiable Privacy-Preserving IoT Monitoring and Settlement",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body style={{ margin: 0, fontFamily: "system-ui, sans-serif", background: "#f5f5f5" }}>
        {children}
      </body>
    </html>
  );
}
