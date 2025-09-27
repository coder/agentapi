"use client"

import {useState, MouseEvent, Dispatch, SetStateAction, useRef} from "react"
import { useDropzone } from "react-dropzone"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "./ui/dialog"
import { Button } from "./ui/button"
import { Upload, X } from "lucide-react"
import JSZip from "jszip";
import { useChat } from "@/components/chat-provider";
import {toast} from "sonner";
import {Checkbox} from "@/components/ui/checkbox";
import path from "path";

interface UploadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  setMessage: Dispatch<SetStateAction<string>>
}

export function UploadDialog({ open, onOpenChange, setMessage }: UploadDialogProps) {
  const [uploadPath, setUploadPath] = useState("")
  const {uploadFiles} = useChat();
  const [filesToUpload, setFilesToUpload] = useState<File[]>([]);
  const filePathsToAppend = useRef<Set<string>>(new Set([]));
  const [disableUploadPath, setDisableUploadPath] = useState(false);


  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDropAccepted: (files: File[]) => {
      setFilesToUpload((oldFiles) => {
        const updatedFiles = oldFiles;
        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-expect-error
        const oldSet = new Set<string>(oldFiles.map((f) => f.relativePath));
        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-expect-error
        files.forEach(file => oldSet.has(file.relativePath) ? {} : updatedFiles.push(file))
        return updatedFiles;
      });
    }
  })

  const cleanup = (open? : boolean) => {
    if (open !== undefined && open) {
      return
    }

    filePathsToAppend.current = new Set<string>([]);
    setUploadPath("")
    setFilesToUpload([]);
    setDisableUploadPath(false);
    onOpenChange(false)
  }

  const handleFilesUpload = async (e: MouseEvent<HTMLButtonElement>) => {
    e.preventDefault()
    let success = true;
    setDisableUploadPath(true);

    try {
      // Create a new JSZip instance
      const zip = new JSZip();

      // Add each file to the zip
      for (const file of filesToUpload) {
        const arrayBuffer = await file.arrayBuffer();

        // This is needed to preserve the paths, for reasons unknown to me webkitRelativePath is empty
        // but relativePath is available but doesn't show up as a valid tye
        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-expect-error
        const pathInZip =  file.relativePath || file.name;
        zip.file(pathInZip, arrayBuffer);
      }

      // Generate the zip blob
      const zipBlob = await zip.generateAsync({ type: "blob" });

      console.log(`Created zip file with ${filesToUpload.length} files, size: ${zipBlob.size} bytes`);

      // Create FormData for upload
      const formData = new FormData();
      formData.append('file', zipBlob, 'upload.zip');
      formData.append('uploadPath', uploadPath);

      // Upload to agent API
      success = await uploadFiles(formData)

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (error: any) {
      success = false;
      toast.error("Failed to zip and upload files:", {
        description: error.message,
      });
    }
    if (success) {
      for (const filePath of filePathsToAppend.current) {
        setMessage(oldMessage =>  oldMessage + ' @"' + path.join(uploadPath, filePath) + '"');
      }
      cleanup()
    } else {
      setDisableUploadPath(false);
    }
  }


  const removeFile = (fileToRemove: File) => {
    setFilesToUpload((prevFiles) =>
      prevFiles.filter(
        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-expect-error
        (file) => file.relativePath !== fileToRemove.relativePath
      )
    );
  };

  return (
    <Dialog open={open} onOpenChange={cleanup}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Upload Files</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <label htmlFor="uploadPath" className="text-sm font-medium">
              Upload Path
            </label>
            <input
              id="uploadPath"
              type="text"
              value={uploadPath}
              onChange={(e) => setUploadPath(e.target.value)}
              className="w-full px-3 py-2 text-sm border rounded-md focus:outline-none focus:ring-1 focus:ring-ring"
              placeholder="Enter upload path..."
              disabled={disableUploadPath}
            />
          </div>

          <div
            {...getRootProps()}
            className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors cursor-pointer ${
              isDragActive
                ? 'border-primary bg-primary/5'
                : 'border-border hover:border-primary/50'
            }`}
          >
            <input {...getInputProps()}/>
            <Upload className="mx-auto h-8 w-8 text-muted-foreground mb-2" />
            {isDragActive ? (
              <p className="text-sm text-primary">Drop the files here...</p>
            ) : (
              <div className="space-y-1">
                <p className="text-sm font-medium">Click to upload or drag and drop</p>
                <p className="text-xs text-muted-foreground">Any file type supported</p>
              </div>
            )}
          </div>

          {filesToUpload.length > 0 && (
            <div className="space-y-2">
              <h4 className="text-sm font-medium">Selected Files (select the checkbox to append @filepath to message)</h4>
              <div className="space-y-1 max-h-32 overflow-y-auto">
                {filesToUpload.map((file, index) => (
                  <div
                    key={index}
                    className="flex items-center justify-between p-2 bg-muted rounded-md text-sm"
                  >
                    {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                    {/*// @ts-expect-error*/}
                    <span className="truncate">{file.relativePath}</span>
                    <div className="flex items-center justify-between gap-2">
                      <Checkbox onCheckedChange={(checked: boolean) => {
                        if (checked) {
                          {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                          {/*// @ts-expect-error*/}
                          filePathsToAppend.current.add(file.relativePath)
                        } else {
                          {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                          {/*// @ts-expect-error*/}
                          filePathsToAppend.current.delete(file.relativePath)
                        }
                      }}/>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => removeFile(file)}
                      >
                        <X className="h-3 w-3" />
                      </Button>
                    </div>

                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={()=> cleanup()}>
              Cancel
            </Button>
            <Button disabled={filesToUpload.length === 0 || uploadPath.length === 0} onClick={handleFilesUpload}>
              Upload
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}


