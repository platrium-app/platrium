import React, { createContext, useContext, useRef, useState } from "react"
import { UploadSource } from "platrium-sdk"
import { usePlatriumSdk } from "./PlatriumSdkContext"

interface UploadContextType {
  triggerUpload: (folderId: string) => void
}

const UploadContext = createContext<UploadContextType | null>(null)

export function useUpload() {
  const context = useContext(UploadContext)
  if (!context) {
    throw new Error("useUpload must be used within an UploadProvider")
  }
  return context
}

export function UploadProvider({ children }: { children: React.ReactNode }) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const targetFolderIdRef = useRef<string | null>(null)
  const client = usePlatriumSdk()
  
  // Track active uploads for future UI
  const [activeUploads, setActiveUploads] = useState<string[]>([])

  const triggerUpload = (folderId: string) => {
    targetFolderIdRef.current = folderId
    fileInputRef.current?.click()
  }

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    const folderId = targetFolderIdRef.current

    if (!file || !folderId) return

    console.log(`Starting upload for ${file.name} to folder ${folderId}`)
    setActiveUploads((prev) => [...prev, file.name])

    try {
      const source = new UploadSource(file.name, file)
      const filesApi = client.files()
      
      const fileId = await filesApi.upload(folderId, source)
      console.log(`Upload complete! Backend file ID: ${fileId}`)
      
      setActiveUploads((prev) => prev.filter((name) => name !== file.name))
    } catch (err) {
      console.error(`Failed to upload ${file.name}:`, err)
      setActiveUploads((prev) => prev.filter((name) => name !== file.name))
    }

    // Reset input so the same file can be uploaded again if needed
    if (fileInputRef.current) {
      fileInputRef.current.value = ""
    }
    targetFolderIdRef.current = null
  }

  return (
    <UploadContext.Provider value={{ triggerUpload }}>
      {children}
      
      {/* Hidden file input for native OS picker */}
      <input
        type="file"
        ref={fileInputRef}
        onChange={handleFileChange}
        style={{ display: "none" }}
      />
      
      {/* TODO: Implement global progress toast component here in the future */}
      {/* 
        {activeUploads.length > 0 && (
          <div className="fixed bottom-4 right-4 bg-background border p-4 shadow-lg rounded-md">
            Uploading {activeUploads.length} file(s)...
          </div>
        )}
      */}
    </UploadContext.Provider>
  )
}
