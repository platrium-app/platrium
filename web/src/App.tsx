import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import RootLayout from "./layouts/RootLayout"
import FolderRootView from "./pages/folder/FolderRootView"
import { UploadProvider } from "./contexts/UploadContext"
import { BreadcrumbProvider } from "./contexts/BreadcrumbContext"

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
              <Route path="*" element={<p>Page Not Found</p>} />
            </Route>
          </Routes>
        </BrowserRouter>
      </BreadcrumbProvider>
    </UploadProvider>
  )
}

export default App
