import SwiftUI

enum MimeTypeIcon {
    static func systemImageName(for mimeType: String?) -> String {
        guard let mimeType = mimeType?.lowercased() else {
            return "doc.fill"
        }
        
        if mimeType.hasPrefix("image/") {
            return "photo.fill"
        } else if mimeType.hasPrefix("video/") {
            return "play.rectangle.fill"
        } else if mimeType.hasPrefix("audio/") {
            return "waveform"
        } else if mimeType.hasPrefix("text/") {
            return "doc.text.fill"
        } else if mimeType == "application/pdf" {
            return "doc.richtext.fill"
        } else if mimeType == "application/zip" || mimeType == "application/x-tar" || mimeType == "application/gzip" {
            return "doc.zipper"
        } else if mimeType.contains("spreadsheet") || mimeType.contains("excel") || mimeType == "text/csv" {
            return "tablecells.fill"
        } else if mimeType.contains("presentation") || mimeType.contains("powerpoint") {
            return "chart.bar.doc.horizontal.fill"
        } else if mimeType.contains("wordprocessing") || mimeType.contains("word") {
            return "doc.text.fill"
        }
        
        return "doc.fill"
    }
    
    static func color(for mimeType: String?) -> Color {
        guard let mimeType = mimeType?.lowercased() else {
            return .gray
        }
        
        if mimeType.hasPrefix("image/") {
            return .purple
        } else if mimeType.hasPrefix("video/") {
            return .pink
        } else if mimeType.hasPrefix("audio/") {
            return .orange
        } else if mimeType == "application/pdf" {
            return .red
        } else if mimeType.contains("spreadsheet") || mimeType.contains("excel") || mimeType == "text/csv" {
            return .green
        } else if mimeType.contains("presentation") || mimeType.contains("powerpoint") {
            return .yellow
        } else if mimeType.contains("word") {
            return .blue
        }
        
        return .gray
    }
}
