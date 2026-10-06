import { BrowserRouter, Routes, Route, Navigate, Outlet } from "react-router-dom"
import { TelescopeIcon } from "lucide-react"
import AuthAwareLayout from "./layouts/AuthAwareLayout"
import FolderRootView from "./pages/folder/FolderRootView"
import { UploadProvider } from "./contexts/UploadContext"
import { BreadcrumbProvider } from "./contexts/BreadcrumbContext"
import { PlaceholderView } from "./components/custom/PlaceholderView"
import DownloadFallbackView from "./pages/DownloadFallbackView"

import HomeView from "./pages/HomeView"
import SharedDrivesView from "./pages/drives/SharedDrivesView"
import SharedWithMeView from "./pages/shared/SharedWithMeView"
import { FilePreviewView } from "./pages/filepreview/FilePreviewCore"

import LoginView from "./pages/LoginView"
import AuthorizeView from "./pages/AuthorizeView"
import DevicesAppsView from "./pages/DevicesAppsView"
import { ServerInfoProvider } from "./contexts/ServerInfoContext"
import { ApolloProvider } from "@apollo/client/react"
import { client } from "./lib/apollo"

import { AuthProvider } from "./contexts/AuthContext"
import { RequireAuth } from "./components/auth/RequireAuth"
import { RequirePermission } from "./components/auth/RequirePermission"
import AdminUsersView from "./pages/admin/users/AdminUsersView"

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

              {/* Native apps and CLIs send users here to approve a sign-in. */}
              <Route path="/authorize" element={<RequireAuth><AuthorizeView /></RequireAuth>} />

              <Route path="/file/:id" element={<FilePreviewView />} />

              {/*
                One layout for every page that can show the app frame, so moving
                between a folder and the rest keeps the sidebar mounted (and its
                panel slide animating). Folders can be shared with anyone who has
                the link, so visitors may try them too; the rest needs a sign-in.
              */}
              <Route element={<AuthAwareLayout />}>
                <Route path="/folder/:id" element={<FolderRootView />} />

                <Route element={<RequireAuth><Outlet /></RequireAuth>}>
                  <Route path="/" element={<Navigate to="/home" replace />} />
                  <Route path="/home" element={<HomeView />} />
                  <Route path="/shared" element={<SharedWithMeView />} />
                  <Route path="/shared-drives" element={<SharedDrivesView />} />
                  <Route path="/settings/devices" element={<DevicesAppsView />} />

                  {/* Admin console. The sidebar switches to its own panel under /admin. */}
                  <Route
                    path="/admin"
                    element={
                      <RequirePermission permission="USERS_READ">
                        <Outlet />
                      </RequirePermission>
                    }
                  >
                    <Route index element={<Navigate to="users" replace />} />
                    <Route path="users" element={<AdminUsersView />} />
                  </Route>
                  <Route
                    path="/rawcontent/*"
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

