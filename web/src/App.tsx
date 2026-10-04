import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { TelescopeIcon } from "lucide-react"
import RootLayout from "./layouts/RootLayout"
import FolderRootView from "./pages/folder/FolderRootView"
import { UploadProvider } from "./contexts/UploadContext"
import { BreadcrumbProvider } from "./contexts/BreadcrumbContext"
import { PlaceholderView } from "./components/custom/PlaceholderView"
import DownloadFallbackView from "./pages/DownloadFallbackView"

import HomeView from "./pages/HomeView"
import { FilePreviewView } from "./pages/filepreview/FilePreviewCore"

import LoginView from "./pages/LoginView"
import { ServerInfoProvider } from "./contexts/ServerInfoContext"
import { ApolloProvider } from "@apollo/client/react"
import { client } from "./lib/apollo"

import { AuthProvider } from "./contexts/AuthContext"
import { RequireAuth } from "./components/auth/RequireAuth"

export function App() {
  return (
    <ApolloProvider client={client}>
      <ServerInfoProvider>
      <AuthProvider>
      <UploadProvider>
        <BreadcrumbProvider>
          <BrowserRouter>
            <Routes>
              {/* Auth Routes */}
              <Route path="/login" element={<LoginView />} />
              <Route path="/login/:alias" element={<LoginView />} />

              <Route path="/file/:id" element={<FilePreviewView />} />
              
              <Route path="/" element={
                <RequireAuth>
                  <RootLayout />
                </RequireAuth>
              }>
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
      </AuthProvider>
      </ServerInfoProvider>
    </ApolloProvider>
  )
}


export default App

