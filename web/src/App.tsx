import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import RootLayout from "./layouts/RootLayout"
import FolderRootView from "./pages/folder/FolderRootView"

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<RootLayout />}>
          <Route index element={<Navigate to="/home" replace />} />
          <Route path="home" element={<p>Home Content</p>} />
          <Route path="folder/:id" element={<FolderRootView />} />
          <Route path="*" element={<p>Page Not Found</p>} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
