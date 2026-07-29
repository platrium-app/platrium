import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "./index.css"
import App from "./App.tsx"
import { ThemeProvider } from "@/components/theme-provider.tsx"
import { TooltipProvider } from "@/components/ui/tooltip"

import initWasm from "platrium-sdk"
import { PlatriumSdkProvider } from "@/contexts/PlatriumSdkContext"

initWasm().then(() => {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <PlatriumSdkProvider defaultUrl="http://localhost:3000/api">
        <ThemeProvider>
          <TooltipProvider>
            <App />
          </TooltipProvider>
        </ThemeProvider>
      </PlatriumSdkProvider>
    </StrictMode>
  )
})
