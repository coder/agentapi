import React from "react";
import { useDropzone } from "react-dropzone";

export interface DragDropProps {
  onFilesAdded: (files: File[]) => void;
  disabled?: boolean;
  children: React.ReactNode;
  className?: string;
}

export function DragDrop({ onFilesAdded, disabled = false, children, className = "" }: DragDropProps) {
  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    noClick: true,
    disabled,
    onDropAccepted: (files: File[]) => {
      onFilesAdded(files);
    },
    multiple: true,
  });

  return (
    <div
      {...getRootProps()}
      className={`relative ${className} ${
        isDragActive && !disabled ? 'border-primary border-2 border-dashed rounded-lg text-center transition-colors' : ''
      }`}
    >
      <input {...getInputProps()} />
      {isDragActive && !disabled && (
        <div className="absolute inset-0 flex items-center justify-center bg-primary/20z-10">
          <p className="text-sm text-primary font-medium">Drop the files here</p>
        </div>
      )}
      {children}
    </div>
  );
}