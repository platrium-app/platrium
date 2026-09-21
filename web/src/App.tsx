import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { TelescopeIcon } from "lucide-react"
import RootLayout from "./layouts/RootLayout"
import FolderRootView from "./pages/folder/FolderRootView"
import { UploadProvider } from "./contexts/UploadContext"
import { BreadcrumbProvider } from "./contexts/BreadcrumbContext"
import { PlaceholderView } from "./components/custom/PlaceholderView"
import DownloadFallbackView from "./pages/DownloadFallbackView"

import HomeView from "./pages/HomeView"

export function App() {
  return (
    <UploadProvider>
      <BreadcrumbProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<RootLayout />}>
              <Route index element={<Navigate to="/home" replace />} />
              <Route path="home" element={<HomeView />} />
              <Route path="folder/:id" element={<FolderRootView />} />
              <Route
                path="rawcontent/*"
                element={<DownloadFallbackView />}
              />
              <Route
                path="*"
                element={
                  <PlaceholderView
                    icon={TelescopeIcon}
                    title="Page Not Found"
                    description="We've looked everywhere and the page you're looking for does not exist or has been moved."
                  />
                }
              />
            </Route>
          </Routes>
        </BrowserRouter>
      </BreadcrumbProvider>
    </UploadProvider>
  )
}

export default App

