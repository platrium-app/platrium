import React, { createContext, useContext, useEffect, useRef, useState } from "react"
import { UploadSource } from "platrium-sdk"
import type { NetTransferEvent } from "platrium-sdk"
import { usePlatriumSdk } from "./PlatriumSdkContext"
import { FileTransferStack } from "@/components/custom/FileTransferStack"


interface UploadContextType {
  triggerUpload: (folderId: string) => void
  cancelTransfer: (transferId: string) => Promise<void>
  clearCompleted: () => void
  transfers: NetTransferEvent[]
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

  const [transfers, setTransfers] = useState<NetTransferEvent[]>([])

  useEffect(() => {
    if (!client) return

    const filesApi = client.files()
    const subscription = filesApi.onTransferEvent((event: NetTransferEvent) => {
      setTransfers((prev) => {
        const existingIdx = prev.findIndex((t) => t.transferId === event.transferId)
        if (existingIdx >= 0) {
          const updated = [...prev]
          updated[existingIdx] = event
          return updated
        } else {
          return [event, ...prev]
        }
      })
    })

    return () => {
      subscription.unsubscribe()
    }
  }, [client])

  const triggerUpload = (folderId: string) => {
    targetFolderIdRef.current = folderId
    fileInputRef.current?.click()
  }

  const cancelTransfer = async (transferId: string) => {
    if (!client) return
    try {
      await client.files().cancelUpload(transferId)
    } catch (err) {
      console.error(`Failed to cancel transfer ${transferId}:`, err)
    }
  }

  const clearCompleted = () => {
    setTransfers((prev) =>
      prev.filter((t) => t.status.type === "Preparing" || t.status.type === "Transferring")
    )
  }



  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    const folderId = targetFolderIdRef.current

    if (!file || !folderId) return

    try {
      const source = new UploadSource(file.name, file)
      const filesApi = client.files()
      await filesApi.upload(folderId, source)
    } catch (err) {
      console.error(`Failed to upload ${file.name}:`, err)
    }

    if (fileInputRef.current) {
      fileInputRef.current.value = ""
    }
    targetFolderIdRef.current = null
  }

  return (

    <UploadContext.Provider value={{ triggerUpload, cancelTransfer, clearCompleted, transfers }}>
      {children}
      
      {/* Hidden file input for native OS picker */}
      <input
        type="file"
        ref={fileInputRef}
        onChange={handleFileChange}
        style={{ display: "none" }}
      />

      {/* Floating Desktop Transfer Manager Stack */}
      <FileTransferStack />
    </UploadContext.Provider>
  )
}


